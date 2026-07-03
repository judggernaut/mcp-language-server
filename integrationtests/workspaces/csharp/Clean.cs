namespace Workspace;

// TestClass is a test class with fields and methods
public class TestClass
{
    public string Name = "";
    public int Age;

    // Method is a method on TestClass
    public string Method()
    {
        return Name;
    }
}

// TestInterface defines a simple interface
public interface TestInterface
{
    void DoSomething();
}

public static class TestConstants
{
    // TestConstant is a constant
    public const string TestConstant = "constant value";
}

public static class Globals
{
    // TestVariable is a static field
    public static int TestVariable = 42;

    // TestFunction is a function for testing
    public static void TestFunction()
    {
        Console.WriteLine("This is a test function");
    }

    // CleanFunction is a clean function without errors
    public static void CleanFunction()
    {
        Console.WriteLine("This is a clean function without errors");
    }
}
